package com.avatar.android.core

import com.avatar.android.core.memory.MemoryAction
import com.avatar.android.core.memory.MemoryVault
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.File

class MemoryVaultTest {

    private lateinit var tempFile: File
    private lateinit var vault: MemoryVault
    private val deviceId = "pixel_10_test_device"

    @Before
    fun setUp() {
        tempFile = File.createTempFile("test_memory", ".json")
        vault = MemoryVault(tempFile, deviceId)
    }

    @After
    fun tearDown() {
        tempFile.delete()
    }

    @Test
    fun testTwoPhaseCommit_StageThenConfirm() {
        // Phase 1: Oral command stages memory
        val cmdRes = vault.parseMemoryCommand("帮我记一下我喜欢喝冰美式")
        assertEquals(MemoryAction.STAGE, cmdRes.action)
        assertNotNull(vault.pendingMemory)
        assertEquals("我喜欢喝冰美式", vault.pendingMemory?.content)

        // Verify NOT yet in persistent storage before confirmation
        assertEquals(0, vault.getAll().size)

        // Phase 2: User confirms
        val saved = vault.confirmPendingMemory()
        assertNotNull(saved)
        assertNull(vault.pendingMemory)

        // Verify now persisted
        val list = vault.getAll()
        assertEquals(1, list.size)
        assertEquals("我喜欢喝冰美式", list[0].content)
        assertEquals(deviceId, list[0].deviceId)
    }

    @Test
    fun testTwoPhaseCommit_StageThenReject() {
        vault.parseMemoryCommand("记住我妈妈的生日是5月2日")
        assertNotNull(vault.pendingMemory)

        vault.rejectPendingMemory()
        assertNull(vault.pendingMemory)
        assertEquals(0, vault.getAll().size)
    }

    @Test
    fun testDeleteMemory_RemovesPermanently() {
        val item = vault.addDirect("我喜欢黑色")
        assertEquals(1, vault.getAll().size)

        val deleted = vault.delete(item.id)
        assertTrue(deleted)
        assertEquals(0, vault.getAll().size)
        assertNull(vault.getById(item.id))
    }

    @Test
    fun testDeviceIsolation_PixelVsMac() {
        vault.addDirect("Pixel独有记忆条目")

        // Create vault with another deviceId on same file
        val macVault = MemoryVault(tempFile, "mac_studio_m2")
        // Mac vault should NOT see Pixel's memories
        assertEquals(0, macVault.getAll().size)

        macVault.addDirect("Mac独有记忆条目")
        assertEquals(1, macVault.getAll().size)
        assertEquals("Mac独有记忆条目", macVault.getAll()[0].content)

        // Reload pixel vault
        val reloadedPixelVault = MemoryVault(tempFile, deviceId)
        assertEquals(1, reloadedPixelVault.getAll().size)
        assertEquals("Pixel独有记忆条目", reloadedPixelVault.getAll()[0].content)
    }
}
